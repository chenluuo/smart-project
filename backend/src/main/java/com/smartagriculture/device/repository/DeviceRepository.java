package com.smartagriculture.device.repository;

import com.smartagriculture.device.domain.Device;
import java.util.Optional;
import org.springframework.data.jpa.repository.JpaRepository;

public interface DeviceRepository extends JpaRepository<Device, Long> {

    Optional<Device> findByDeviceCode(String deviceCode);

    Optional<Device> findBySerialNo(String serialNo);
}
