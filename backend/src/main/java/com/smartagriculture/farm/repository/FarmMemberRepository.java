package com.smartagriculture.farm.repository;

import com.smartagriculture.farm.domain.FarmMember;
import java.util.List;
import org.springframework.data.jpa.repository.JpaRepository;

public interface FarmMemberRepository extends JpaRepository<FarmMember, Long> {

    List<FarmMember> findByUserId(Long userId);
}
